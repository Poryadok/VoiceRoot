# ExecPlan: protected Role override target validation

## Purpose

BE-065 prevents a Role override setter from writing an arbitrary Chat or Voice
room UUID. Chat and Space validate resources they own through narrow protected
RPCs; Role retains actor authorization and performs its final authorization and
write in one fenced transaction.

## Context

- Accepted direction: TTenrec `voice` / BE-065 comments c2-c3,
  `BE-065-target-v1`. This document makes that decision repository-visible;
  it does not claim the target is implemented.
- Canon: [roles](../features/roles.md), [Role](../microservices/role-service.md),
  [Chat](../microservices/chat-service.md), [Space](../microservices/space-service.md),
  [Phase-0](../ARCHITECTURE_REQUIREMENTS.md), [data model](../DATA_MODEL.md),
  [testing](../TESTING.md).
- Current setters: `role/internal/grpcsvc/ordinary_scope.go`, `roles.go`,
  `roles_manage.go`. The ordinary wrapper holds a Role transaction across its
  callback; provider calls must therefore occur outside that callback.
- Current owner listeners are specialized for other callers. Their existing
  Social/Search/GIS allow-lists must not be broadened into this authority.

## Scope

In scope: additive Chat/Space proto methods, owner-store validation, exact Role
workload verification on protected TLS/mTLS listeners, signed Role clients,
both setter entrypoints, focused negative/security/store/concurrency tests and
configuration/wiring documentation. No public Gateway route is added.

Removal remains local authorized cleanup. No distributed foreign key,
reservation, positive cache, new deletion event, public ownership/deletion
activation, permission-bit redesign or acceptance/deployment is included.

## Milestones

- [x] Accepted target contract in all three owning service docs.
- [ ] Additive proto/bindings and owner handlers reject invalid/ineligible targets.
- [ ] Dedicated protected owner listeners and signed Role clients fail closed.
- [ ] Both setters use preflight -> unlocked owner reads -> fenced reauthorization/write.
- [ ] Required negative, real-store and transaction/lock-order checks pass.
- [ ] Independent security/concurrency review and exact-head hosted checks pass.

## Detailed Steps

1. Add package-local `ValidateRoleOverrideTargetRequest/Response` to ChatService
   and SpaceService. Chat request fields are `space_id=1`, `chat_id=2`. Space
   request fields are `space_id=1`, optional `voice_room_id=2`; response is empty.
   Document security beside the RPC; regenerate all committed consumers using
   `make buf-generate-all` and `make buf-generate-dart`.
2. Add owner store reads with existing authoritative ordinary lifecycle boundary.
   Chat accepts only active group/channel rows in the requested Space. Space
   validates live Space and, when present, its own live room in that Space.
   UUIDs must be canonical and non-nil; present-empty optional room is invalid.
   Provider methods never call Role or an ACL-enriched lookup.
3. Test handler-level verified Role identity independently of interceptors, then
   bind each dedicated protected listener to only its one exact method. Reuse
   Phase-0 verifier/JWKS/replay/TLS primitives without widening existing listeners.
   Test actual TLS/JWKS/replay failure and ordinary-listener denial before store.
4. Add signed clients using Role's issuer, fresh canonical request ID and JTI,
   exact target audience/method and deterministic full-request hash (including
   optional presence). Replace outgoing metadata rather than forwarding user
   credentials. Bound calls by a per-call deadline and parent cancellation.
5. Refactor shared local setter authorization without changing its semantics.
   Run a short fenced authorization-only transaction, release it, perform Space
   and Chat checks for chat overrides or room-bearing Space check for voice,
   then run the existing fenced mutation and repeat current actor/role/hierarchy
   checks in that final transaction. Buffer events until successful commit.
6. Validate no-write/error branches and permission/freeze changes between
   preflight and final commit. Real PostgreSQL plus bounded network fakes must
   prove owner calls hold no Role transaction/advisory lock and do not deadlock
   with provider operations that need Role decisions. Validate removal still
   works after target deletion and never contacts owner validation.
7. Run impacted Go tests/vet/lint, proto lint/format/breaking and generated parity.
   Obtain independent high-risk source review and required hosted checks, then
   integrate through normal PR gates. Keep actual DEV/STG/acceptance separate.

## Validation

Required case matrix (no PASS is implied):

- Owner: group/channel and matching room success; absent/DM/standalone/foreign/
  deleted target indistinguishable NOT_FOUND; missing versus empty optional
  presence; malformed/nil/noncanonical IDs; all lifecycle/ownership fences,
  terminal retirement and unavailable/corrupt state; restore requires fresh read.
- Transport: both exact RPCs exercise valid Role principal; no/duplicate bearer
  or request ID; raw identity metadata; changed body/optional presence; wrong
  audience/RPC/signature/time/kid/JTI; other authenticated callers; revoked or
  unknown client certificate; JWKS hard expiry and replay-store failure.
  Direct handler calls still require verified Role identity; no outgoing ACL call.
- Role: both owner calls for Chat and one room-bearing call for Voice; absent
  clients/config; owner error/timeout/auth failure/unimplemented/malformed success;
  parent cancellation; no changed rows or events on failure; current role scope,
  permissions/hierarchy and fences rechecked in final transaction.
- Concurrency: permission revoke, role scope change, freeze/retirement between
  phases deny; provider calls outside Role locks; accepted delete-after-read
  race may leave inert logical row but cannot authorize missing resource or a
  new UUID; no cached allow across retry; removal never requires live target.

Commands: affected `go test -short ./...`, focused non-short PostgreSQL/security
tests, `go vet ./...` and selected `golangci-lint` per [TESTING](../TESTING.md);
`make buf-ci`, `make buf-breaking`, `make buf-go-pb-check` and
`make buf-dart-check` when proto/generated code changes. Respect documented
local runtime restrictions; hosted actual non-skipped proof can cover unavailable
PostgreSQL/TLS dependencies. Do not rerun unrelated whole-repository checks.

## Progress

- [x] Accepted TT c2-c3 inspected; real owner handoff e5 received by Рудеус-39.
- [x] Isolated treehouse34 acquired at base `698d4f064f032ea74abf3ede2017100b26c8ee69`.
- [x] Canonical contract in owning docs; all four changed documents pass
  `markdown-link-check@3.12.2` with the repository configuration.
- [ ] Runtime implementation and verification remain open.

## Decisions

The accepted target is point-in-time validation. Deletion between provider read
and Role commit may leave an inert logical reference; it neither resurrects the
resource nor grants access. IDs cannot be reused/reparented; group/channel type
conversion preserves identity. No cross-database SQL or provider reservation is
introduced. See the owning service contract for exact errors and caller binding.

## Risks And Follow-Ups

This is a cross-service ACL/lock-order change and requires independent review.
Existing protected listeners cannot silently acquire a new caller. Fresh owner
success is not a reusable capability. Runtime acceptance and deployment require
their own actual authority and versioned evidence; a source PR does not supply it.
