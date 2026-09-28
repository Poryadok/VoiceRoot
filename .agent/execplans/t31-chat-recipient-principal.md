# ExecPlan: T31 Chat recipient principal and managed resource slice

## Goal and scope

Implement the first recipient-owned T31 vertical in Chat: GIS may create an
application-managed group chat and replace its membership only through exact,
request-bound GIS service-principal RPCs. Chat stores the chat, application/env
ownership marker, operation receipts, and membership in `chat_db`. GIS has no
Chat database access. A managed chat has no human creator, owner, or admin;
all member rows have role `member`. Existing player RPCs cannot mutate a
managed roster.

This is a Chat slice of T31, not completion of GIS orchestration, Voice room
provisioning, grants/entitlements, compensation, or reconciliation.

## Sources read

- `docs/PLAN.md` — game integrations run as a separate sprint on
  `codex/game-sdk-federation-docs`.
- `docs/testing/game-integrations-exec-plan.md` — T30 persists resource keys,
  operation hashes and transitions; T31 orchestrates Chat, Voice, grants,
  receipts and crash recovery.
- `docs/architecture/game-integration-api.md` — operation IDs, exact request
  hash binding, recipient-owned resources, and no caller-selected player
  identity.
- `docs/ARCHITECTURE_REQUIREMENTS.md` — RS256 principal, exact audience/RPC/
  request hash/request ID, replay defense, TLS, allowlist, fail-closed errors.
- `docs/microservices/chat-service.md` and `docs/TESTING.md`.
- CodeGraph: `ChatGRPC.CreateChat`, `VoiceGRPC.StartCall`, `pkg/principal`
  issuer/verifier, Chat/Voice service registration. Its index reported frozen
  auto-sync; source was checked on disk.

## Acceptance criteria

1. Two recipient RPCs are isolated from player RPC semantics: create one
   managed chat and synchronize its member set.
2. Each RPC requires `operation_id`; verifier checks a short-lived RS256 GIS
   service credential bound to exact `chat` audience, exact full method, the
   same operation ID and deterministic hash of the complete request.
3. Replay `jti` is recorded atomically with a short TTL. Absent, duplicate,
   malformed, wrong-bound, replayed, or unavailable proof fails before store
   access. A valid principal outside the explicit RPC allowlist is denied.
4. Durable Chat receipts make an identical operation replay return the original
   result; a changed body with the same operation ID conflicts. Stable
   `(application_id, environment_id, external_chat_key)` cannot create a second
   chat after transport receipt expiry.
5. Managed chat ownership is application-scoped inside Chat. No player is
   assigned owner/admin or accepted as creator; public player methods cannot
   add, remove, leave, or otherwise alter its roster.
6. Protected GIS methods are exposed on a separate TLS-only listener. No raw
   identity headers are accepted. Ordinary Chat listener does not expose these
   methods.
7. Focused tests cover principal rejection before persistence, exact binding,
   replay, request conflict, stable-key reuse, and player roster denial.

## Red-green sequence

1. Add contract tests for the proto method names/fields and service-listener
   allowlist; run them red.
2. Add store tests for managed creation and member sync replay/conflict,
   resource-key uniqueness, and ownerless membership; run them red.
3. Implement Chat schema/store and the narrow managed RPCs; run focused Chat
   tests green.
4. Add verifier and isolated TLS listener; test invalid/missing/replayed/wrong
   audience/RPC/request binding and allowed calls; run green.
5. Add ordinary player-path regression tests for managed-chat roster denial;
   run green, refactor, and rerun focused tests.
6. Update the API and Chat service contract; run `make buf-ci`, the affected Go
   package tests, and migration checks from `docs/TESTING.md`.

## Expected files

- `protos/voice/chat/v1/chat.proto` and generated Chat Go bindings.
- `src/backend/chat/main.go`, new narrow GIS gRPC adapter/verifier runtime,
  and Chat store/service files.
- `src/backend/migrations/chat_db/*`.
- Chat store and gRPC tests.
- `docs/architecture/game-integration-api.md`,
  `docs/microservices/chat-service.md`, and this plan.

## Risks and bounded follow-up

- This receiver cannot know whether GIS supplied an authorized application/env;
  GIS remains the sole authority for that selection. Chat binds all operations
  and resource keys to the supplied app/env tuple and never accepts a player
  actor.
- Existing Chat reads and role-management paths assume a creator/owner in some
  queries. Managed chats must be excluded from each player mutation path and
  represented without inventing a synthetic profile.
- T31 still needs Voice-owned provisioning, GIS durable orchestration stages,
  retry/reconciliation, and the full integration test matrix.

## Status

- [x] Read T31 dependencies and exact Chat ownership gap.
- [x] RED store tests reproduced before implementation; recipient/runtime
  negative tests cover replay, wrong RPC/request ID/hash, raw headers, partial
  configuration, HTTP JWKS downgrade, and client-certificate requirement.
- [x] Chat managed resource, atomic member replacement, operation receipts,
  and ordinary player mutation guards implemented.
- [x] Request-bound verifier and isolated mTLS listener wired in Chat main;
  staging/production secret/port/network-policy wiring remains open.
- [x] Docs and focused verification complete: Chat grpcsvc/store and
  gisprincipal tests, Chat main compile, Buf Go/Dart generation parity, and
  buf lint/format checks.
- [ ] Final source review and handoff evidence.
