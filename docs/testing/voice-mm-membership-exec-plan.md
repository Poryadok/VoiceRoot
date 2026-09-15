# Voice membership schema for authoritative matchmaking

## Purpose

Persist the room kind, immutable purpose and verified membership identity needed
by the accepted A3 Voice party snapshot. This is a source-disabled storage step;
it does not claim that party lookup, roster events or media handlers are active.

## Context

Sources: `docs/todo/backend.md` A3 MM ↔ Voice contract; `docs/features/matchmaking.md`
party semantics; `docs/features/voice-chat.md` reconnect; `docs/microservices/voice-service.md`
R22 lifecycle; `docs/DATA_MODEL.md` account/profile and session epoch;
`docs/DATA_STORES.md`; `docs/ARCHITECTURE_REQUIREMENTS.md` principals;
`docs/PLAN.md`; `docs/TESTING.md`; `docs/CONTRIBUTING.md`.

The R22 PostgreSQL store supports only Space room IDs and media authority.
It does not persist a membership account, session epoch or reconnect deadline.
The existing R22 callers and `CheckSchema` remain compatible with migration 1.

## Scope

- Add migration `000003_matchmaking_membership` and a separately gated store read
  model; keep the original lifecycle interface unchanged.
- Room kinds: `voice_room`, `call`, `group_voice`. Space rooms require their
  Space/logical-room pair and no chat; calls/groups require a chat and no Space
  pair. Purpose is `ORDINARY` or `MATCH_SQUAD`. Match rooms additionally retain
  immutable match owner, creation operation, manifest hash and receipt identity;
  they use `group_voice` and require a Chat UUID and Chat creation receipt ID.
  These fields are creation-time evidence only.
- Membership identity consists of nonzero account ID and positive session epoch.
  Known states are `JOINING`, `JOINED`, `RECONNECTING`, `LEAVING`, `LEFT`, `EJECTED`.
  `RECONNECTING` requires both start and deadline, strictly positive interval at
  most 30 seconds. All other states have no reconnect interval. Space-specific
  access and Role epochs remain required for Space memberships and become NULL
  for call/group memberships; room-kind guards enforce that distinction.
- Existing membership rows keep NULL identity/state together. No account or epoch
  is inferred from actor, profile, media epoch, Redis or a room name. New reads
  report unknown rows as an invariant failure, not SOLO. A known identity/state
  cannot be cleared back to legacy unknown.
- New storage reads are not authorization or an MM snapshot: current epoch
  verification and the transactional roster/version/event boundary remain future
  activation prerequisites. The schema does not infer session validity from media
  token expiry.
- No proto, transport, registration, Redis fallback, grant issuance or worker activation.

## Milestones

- [ ] Docs and independently authored RED migration/model tests.
- [ ] Additive schema and gated reads pass those tests.
- [ ] Independent security/concurrency review and affected checks.
- [ ] Hosted PostgreSQL verification and PR merge.

## Detailed steps

1. Add migration tests for unknown legacy rows; all room shapes; immutable purpose
   and match binding; nonzero/paired account+epoch; all membership states; bounded
   reconnect; failed rollback with expanded data; successful rollback for legacy
   rows; old lifecycle schema check. Use existing PostgreSQL 16 testcontainers.
2. Add a short-capable source check for missing migration RED and unit/read-model
   negative tests; then implement migration and store reads. New reads return
   storage unavailable without migration 3 and preserve not-found separately.
3. Run scoped tests after each step, then Go short tests/vet/pinned lint. Run
   PostgreSQL tests locally if Docker is available, otherwise run hosted CI with
   a focused integration job that cannot silently skip the new tests.
4. Review diff, migration constraints and rollback locking independently. Push
   each commit promptly; integrate via PR merge commit after required CI.

## Validation

- `go test -short ./...`, `go vet ./...`, pinned `golangci-lint run ./...`.
- `go test ./internal/roomlifecycle -run '^TestMatchmakingMembership' -count=1`.
- Real PostgreSQL migration/read tests, no skips; old R22 integration regressions.
- No activation edits in `main.go`, gRPC handlers or Redis compatibility code.

## Decisions

- NULL legacy state is a migration marker, not a seventh product lifecycle state.
- Room purpose and creation binding are immutable in PostgreSQL, so an ordinary
  room cannot be relabelled as a match room by a later update.
- DOWN takes exclusive locks and refuses to discard known membership identity,
  non-Space room shape or match ownership. Legacy-only data is reversible.
- Storage read models are concrete internal DTOs, not proof of authenticated
  identity or a replacement for protected RPC verification.

## Progress

- [x] Read canonical contracts and confirmed #385/#386 merged.
- [x] Acquired isolated treehouse slot 9; no overlapping schema owner.
- [x] Model RED: focused short test fails on absent DTO/constants/store API.
- [ ] Migration RED/implementation/review/hosted verification.

## Risks and follow-ups

All old writers still create NULL legacy membership state. Activation must first
replace those writers with verified identity and atomic roster/outbox lifecycle
mutations. Ordinary call/group lifecycle and squad creation receipts need their
own protected writers. No existing opaque row is automatically promoted.
After expanded rows exist, application rollback can keep the additive schema;
schema rollback is deliberately refused until a separately authorized migration
preserves the new evidence.
