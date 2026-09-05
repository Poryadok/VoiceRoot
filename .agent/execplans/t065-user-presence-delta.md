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
- [ ] Review RED tests with a fresh agent and resolve critical findings.
- [ ] Add the additive proto fields, regenerate stubs, and implement the smallest
  enum-transition comparison/publish change (GREEN).
- [ ] Commit implementation and generated artifacts (GREEN).
- [ ] Review implementation with a fresh agent and run focused/proto validation.

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
5. Regenerate all Go and Dart protobuf artifacts only with the repository Make
   targets; stop rather than downloading dependencies if a tool needs network.
6. Read the prior live snapshot before the unconditional upsert; after a
   successful upsert publish only when its previous enum differs from the
   normalized incoming enum.
7. Run focused User tests, `buf` format/lint/breaking against local `master`,
   inspect generated copies and final diff, then obtain a fresh implementation
   review.

## Validation

- [ ] `go test ./internal/grpcsvc ./internal/store` in `src/backend/user`
- [ ] `buf format -d --exit-code protos`
- [ ] `buf lint protos`
- [ ] local-master protobuf breaking check using the repository target
- [ ] generated Go/Dart artifact check using the repository target

## Progress

- [x] Documentation, code, and test seams identified.
- [x] RED tests and documentation prepared; offline RED observed with `GOPROXY=off; go test ./internal/grpcsvc ./internal/store`: the delta recorder no longer satisfies the three-argument publisher interface and generated `PresenceChange` lacks `OldStatus` / `NewStatus` fields and getters. `GOPROXY=off; go test ./internal/store` passes without download errors.
- [ ] RED test review accepted.
- [ ] GREEN implementation and generated stubs committed.
- [ ] GREEN review and final validation accepted.

## Decisions

- Preserve `PresenceChange.status` as the current/new value for backward
  consumers; add `old_status` and `new_status` rather than repurposing a wire
  field.
- Use the stored enum, not optional game/custom/call/last-seen fields, as the
  transition predicate. The unconditional upsert remains before any best-effort
  event publish.

## Risks And Follow-Ups

- Reading then writing Redis is not a distributed compare-and-set; concurrent
  heartbeats can independently observe the same old enum. This task preserves
  the current store model and does not invent an atomic transition protocol.
- Existing Realtime consumers continue to consume legacy `status`; migrating
  consumers to delta fields is separate work.
