# ExecPlan: A3 fixture-only matchmaking contracts

## Purpose

Provide deterministic Matchmaking Service contract evidence for concurrent search
lifecycle operations and cleanup before the A2 roster/session-event integration
exists.  The later A3 end-to-end acceptance slice consumes these tests and the
small adapter seam; this plan does not claim that A3 is active or complete.

## Context

- `docs/PLAN.md` A3 defines idempotent concurrent join/leave/cancel and cleanup
  as its DoD, while making A2 roster/session events a dependency.
- `docs/features/matchmaking.md` requires a voice-party search to reset when its
  roster changes, and states that completed temporary squads are cleaned up.
- `docs/microservices/matchmaking-service.md` assigns search sessions, matcher
  queues, temporary-match resources, and their timeout/cleanup behavior to
  Matchmaking Service.
- `docs/todo/backend.md` records that party/voice-derived matchmaking is absent;
  `PartyStore` is a stub and `StartSearch` is solo-only.
- The user explicitly authorizes only a fixture/provider contract preparation
  slice ahead of full A2, for later A3 acceptance.

## Scope

- In: deterministic in-memory/provider fixtures and service tests for a
  concurrent terminal lifecycle race, duplicate lifecycle request idempotency,
  and exactly-once cleanup of fixture-provisioned temporary resources.
- Out: live Voice/A2 roster or session events, client-supplied party membership,
  party snapshot production integration, temp voice/chat creation, new public
  RPCs, migrations, and compose E2E.
- Documentation gap: the feature docs describe the eventual A2-backed flow but
  not a pre-A2 provider interface.  This PR keeps the interface package-private
  and test-only-facing, so it does not establish product behavior.

## Milestones

- [x] Write RED fixture-contract tests.
- [x] Add the smallest package-local implementation that passes each contract.
- [ ] Run focused and module checks; review the final diff.
- [ ] Commit, push, open and merge a CI-green PR; verify `master` and return the lease.

## Detailed Steps

1. Inspect the existing search, matcher, timeout, store, and test seams.
2. Add focused tests under `src/backend/matchmaking/internal/` using deterministic
   fixtures only.  Demonstrate each test fails before its production change.
3. Add the minimum internal provider/coordinator behavior for one contract at a
   time: RED, GREEN, then a behavior-preserving refactor if needed.
4. Run `CGO_ENABLED=0 go test ./...` and the affected lint command from
   `docs/TESTING.md`; update this plan with observed results.
5. Inspect paths and diff, then use the repository's normal commit/push/PR/CI
   merge workflow.  Do not activate any A2 dependency in the process.

## Validation

- [ ] Focused new fixture-contract tests pass with `-count=1`.
- [ ] `CGO_ENABLED=0 go test ./...` in `src/backend/matchmaking` passes.
- [ ] `golangci-lint run ./...` in `src/backend/matchmaking` passes, if locally available.
- [ ] Hosted PR checks, including `ci-gate`, pass before merge.

## Progress

- [x] Read product, service, testing, contribution, plan, and backlog sources.
- [x] Confirmed clean Treehouse lease from base `c46a5bc8`.
- [x] RED: `TestCompleteMatch_FinalLeaveCleansFixtureSquadOnce` initially failed
  to compile because `MatchmakingGRPC` had no cleanup seam.
- [x] GREEN: an internal nullable `SquadCleanup` provider is invoked only when
  `CompleteMatchLeaveWithTransition` reports the durable final transition.
- [x] The existing deterministic concurrent-final-retry test also asserts one
  cleanup call; neither test attaches a roster/session event source.
- [x] `CGO_ENABLED=0 go test -short ./... -count=1` passes in Matchmaking.
- [ ] Full fixture-contract tests require PostgreSQL testcontainers and are
  blocked locally by Docker's Windows rootless error; hosted CI remains required.

## Decisions

- Fixture-only, package-local boundary: preserves the documented A2 dependency
  and cannot accidentally authorize a client party payload.
- No party or voice context implementation: `docs/todo/backend.md` explicitly
  identifies that as unavailable pending the authoritative Voice flow.
- Cleanup call errors are logged when a logger is configured but do not roll
  back the already durable terminal match state. Retry/reconciliation policy is
  deferred to the real A3 provider rather than invented in this fixture seam.

## Risks And Follow-Ups

- This is preparation evidence only; it must be paired with A2 roster/session
  integration and multi-client temporary chat/media E2E before A3 acceptance.
- The concrete fixture seam must reuse existing Matchmaking test conventions and
  must not create a misleading production fallback when its provider is absent.
