# ExecPlan: T-065 user presence transition delta

## Purpose

`user.presence_changed` becomes a useful transition event: it carries the prior
and current canonical presence statuses, emits only when the status enum changes,
and leaves every heartbeat responsible for refreshing live-presence and
last-seen TTLs.

## Context

- Docs: `docs/features/presence.md` (heartbeat and event contract),
  `docs/microservices/user-service.md` (User heartbeat and `user.events`),
  `docs/CONTRACT_MATRIX.md`, `docs/TESTING.md`, and `docs/CONTRIBUTING.md`.
- Contract: `protos/voice/events/v1/jetstream_events.proto` currently has only
  `PresenceChange.status`; committed Go and Dart stubs mirror it.
- Code: `src/backend/user/internal/grpcsvc/user_presence.go` upserts Redis then
  publishes every time; `internal/userevents/jetstream.go` publishes the legacy
  single status field; `internal/store/presence.go` owns the session and
  last-seen TTL writes.
- Constraints: additive protobuf tags only; regenerate every committed Go/Dart
  copy through repository commands; no network or Docker. The worktree starts
  from `a3b06b52` on `codex/t065-user-presence-delta`.

## Scope

- In: documented delta semantics, proto fields, generated stubs, focused User
  Service tests, and minimal comparison/publisher plumbing.
- Out: Realtime privacy/fan-out redesign, durable PostgreSQL last-seen,
  offline-transition detection, and changes to existing consumers beyond
  preserving the legacy `status` value.
- Documentation gap resolved here: first observation has no prior live enum, so
  its `old_status` is the empty string and `new_status` is canonical current
  status.

## Milestones

- [x] Read the governing documentation and inspect current User presence flow.
- [x] Commit documentation plus failing transition-contract tests (RED).
- [x] Review RED tests with fresh agents, remove Docker/publisher/tag-coverage blockers, and obtain a clean re-review.
- [x] Add the additive proto fields, regenerate stubs, and implement the smallest
  enum-transition comparison/publish change (GREEN).
- [x] Commit implementation and generated artifacts (GREEN).
- [x] Review implementation with fresh agents and run focused/proto validation.

## Detailed Steps

1. Document that status comparison is enum-based, a first live observation uses
   empty `old_status`, and same-status heartbeats still perform both Redis writes.
2. Add tests that require first-observation and enum-transition deltas, suppress
   same-enum events despite changed non-status presence fields, and prove the
   heartbeat still refreshes activity/TTLs.
3. Run the focused test package before implementation and record the expected
   failure caused by the absent delta contract/plumbing.
4. Add `old_status` and `new_status` using new protobuf tag numbers while keeping
   the legacy `status` field populated with `new_status` for existing consumers.
5. Regenerate all Go and Dart protobuf artifacts with repository Buf templates
   and the approved mechanical mirror; stop rather than downloading
   dependencies if a tool needs network.
6. Atomically read the prior live snapshot and refresh the session/last-seen
   keys in Redis; after the successful write publish only when the returned
   prior enum differs from the normalized incoming enum.
7. Run focused User tests, `buf` format/lint/breaking against local `master`,
   inspect generated copies and final diff, then obtain a fresh implementation
   review.

## Validation

- [x] Focused transition tests in `internal/grpcsvc`, `internal/store`, and
  `internal/userevents` with `GOPROXY=off`
- [x] `buf format -d --exit-code protos`
- [x] `buf lint protos`
- [x] Local-master protobuf breaking check using the repository target
- [x] Generated Go/Dart artifact check using repository Buf templates and the
  approved Windows mechanical Go mirror
- [x] `GOPROXY=off; go test -race ./internal/store -run
  TestPresenceStore_ConcurrentUpsertsReturnLinearizablePreviousStatus`

## Progress

- [x] Documentation, code, and test seams identified.
- [x] RED tests and documentation prepared; offline RED observed with `GOPROXY=off; go test ./internal/grpcsvc ./internal/store`: the delta recorder no longer satisfies the three-argument publisher interface and generated `PresenceChange` lacks `OldStatus` / `NewStatus` fields and getters. `GOPROXY=off; go test ./internal/store` passes without download errors.
- [x] Initial RED review found that an integration helper would require prohibited Docker and that legacy publisher/tag compatibility was not directly asserted.
- [x] Replaced it with a no-Docker transition helper matrix, direct publisher-envelope RED test, and additive descriptor assertions; focused offline RED now fails only on absent helper/contract seams.
- [x] Fresh RED re-review accepted: no blocking findings; `GOPROXY=off; go test ./internal/grpcsvc ./internal/store ./internal/userevents` fails only at the intended absent helper/generated contract seams.
- [x] GREEN implementation adds proto tags 3/4, regenerates the three Go and
  two Dart event artifacts, compares the stored enum before publishing, and
  preserves the unconditional Redis upsert for every heartbeat.
- [x] Native cached generation completed without Docker or network. The Go
  mirror matched 94 files across 48 package destinations by SHA-256 before four
  unrelated pre-existing generated-stub drifts were precisely reversed; the
  final semantic generated diff is exactly the three Go event copies and two
  Dart event files.
- [x] Focused tests passed with `GOPROXY=off`: three grpcsvc transition/proto
  tests, the miniredis same-status TTL/activity test, and the envelope test.
  The broad grpcsvc package run was stopped after an unrelated existing test
  ran silently beyond the bounded check; no Docker was started.
- [x] `buf format`, `buf lint`, and `buf breaking` against local `master` pass.
- [x] GREEN implementation and generated stubs committed.
- [x] Initial GREEN review found a P1 stale-snapshot race: separate `Get` and
  `Upsert` calls could publish an incorrect old enum or suppress a later
  transition under concurrent heartbeats.
- [x] Correction RED commit `62961446` adds a concurrent same-profile upsert
  test that requires a linearizable prior snapshot and retains the
  first-observation assertion.
- [x] Correction GREEN commit `81776970` moves snapshot/write/TTL refresh to
  one Redis Lua operation. `GOPROXY=off; go test -race ./internal/store` and
  the focused grpcsvc/userevents tests pass; a fresh review also passed the
  concurrent test 20 times without P0-P2 findings.
- [x] The initial local-master `buf breaking` passed after the additive proto
  change. A repeated check after the code-only atomic correction was stopped
  after its local Git child exceeded the 60-second bound; no proto or generated
  artifact changed after the passing check.

## Decisions

- Preserve `PresenceChange.status` as the current/new value for backward
  consumers; add `old_status` and `new_status` rather than repurposing a wire
  field.
- Use the stored enum, not optional game/custom/call/last-seen fields, as the
  transition predicate. A Redis Lua operation atomically returns the prior
  live snapshot while preserving the unconditional session/activity/last-seen
  refresh before any best-effort event publish.
- Keep the transition-decision unit test Docker-free: `UpdatePresence` depends
  on the concrete profile store, while the exact previous-snapshot-to-event
  decision is independently deterministic and the miniredis store test proves
  same-enum heartbeats still refresh activity and both TTLs.

## Risks And Follow-Ups

- Redis Lua serialization is scoped to the two presence keys and avoids a
  distributed lock; if the atomic write fails, User returns an internal error
  and does not publish a transition.
- Existing Realtime consumers continue to consume legacy `status`; migrating
  consumers to delta fields is separate work.
