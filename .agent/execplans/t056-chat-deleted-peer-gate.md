# ExecPlan: T-056 Chat deleted-peer gate

## Purpose

Make Chat's A1 account soft-delete boundary safe: a fresh `ListChats` snapshot
must not reveal a DM whose peer account is soft-deleted, and `CreateDM` / `GetDM`
must reject opening a DM with that peer. The eventual result is fail-closed when
the Auth or User dependencies needed to decide deletion are unavailable. This
plan intentionally separates the Chat boundary from Messaging send enforcement
and Flutter's local-history marker.

## Context

- Product contract: `docs/PLAN.md` A1 and `docs/features/auth-and-contacts.md`
  “Что видят другие после удаления аккаунта”. They require hiding the DM in
  fresh snapshots while already-loaded history remains a client concern.
- Data/service boundary: `docs/DATA_MODEL.md` §3 and
  `docs/microservices/chat-service.md`; Auth owns account soft delete and Chat
  owns the inbox snapshot.
- Error convention: `docs/ARCHITECTURE_REQUIREMENTS.md` § gRPC errors permits
  standard `UNAVAILABLE`; existing Chat role S2S handling maps an unavailable
  dependency to `codes.Unavailable`.
- Existing seam: `ChatGRPC.DeletedAccounts`, `filterDeletedPeerDMRows`, and
  Auth's `FilterDeletedAccountIDs` S2S RPC. Current filtering can return
  unfiltered rows on dependency failure and `ensureDM` has no deleted-account
  gate.

## Scope

- In: `src/backend/chat/internal/grpcsvc` tests and the later minimal Chat
  implementation for `ListChats`, `CreateDM`, and `GetDM`.
- Out: Auth deletion/session lifecycle, Messaging `SendMessage`, Gateway
  mapping changes, Realtime events, Flutter markers/cache/UI, global
  tombstones, account erasure, and restore.
- Documentation gap: the A1 docs define hiding fresh snapshots but do not
  prescribe pagination after post-query filtering. The implementation must
  preserve the documented visible result without adding a public API field.

## Milestones

- [x] Confirm docs, S2S RPC names, and local error conventions.
- [x] Add RED tests for the Chat boundary only.
- [ ] Independently review RED tests; do not write production code before it
  passes review.
- [ ] Implement the smallest fail-closed Chat changes.
- [ ] Run focused and neighbouring Chat tests, then broader service checks.

## Detailed Steps

1. Seed a caller, deleted peer, active peer, DM rows, and a non-DM row using
   the existing Chat Postgres/test-server helpers.
2. Add integration tests showing a deleted-peer DM is absent from `main`,
   `requests`, `archive`, and folder listings; active DM/group/channel remain.
3. Add pagination tests where the first raw page consists only of deleted-peer
   DMs and a later raw page has an active row; assert the public page sequence
   neither leaks nor strands the active row behind an empty successful page.
4. Add dependency-failure tests for absent deleted-account checker, Auth check
   failure, and profile-to-account lookup failure. Each must return
   `codes.Unavailable`, rather than a successful unfiltered or empty snapshot.
5. Add `CreateDM`/`GetDM` tests for a deleted peer: both must return
   `codes.PermissionDenied` with a privacy-safe message. Repeat the calls to
   prove idempotent/repeated attempts do not create or expose the DM.
6. In the green phase, centralize deleted-peer resolution in Chat and wire it
   before `EnsureDM`; preserve ordinary active-peer, group, and channel paths.
7. Run `go test ./internal/grpcsvc -run DeletedPeer` and relevant neighbouring
   `ListChats`/DM tests from `src/backend/chat`, then `CGO_ENABLED=0 go test
   ./...` before requesting implementation review.

## Validation

- [x] Compile-only focused check passes under `-short`; the full integration
  attempt is currently blocked before test bodies by the shared Docker/
  Testcontainers `5432/tcp not found` infrastructure failure. Static review
  confirms the fail-open/no-gate assertions are RED against current code;
  cursor continuation is a baseline-green guard.
- [ ] Focused Chat tests green after implementation.
- [ ] `CGO_ENABLED=0 go test ./...` from `src/backend/chat` green after
  implementation.
- [ ] Final diff contains only Chat tests/implementation and this plan.

## Progress

- [x] Plan written before test edits.
- [x] Test-only RED implementation and focused compile run.
- [x] Independent test review.
- [ ] Green implementation (future turn).

## Decisions

- Dependency-decision failure is `UNAVAILABLE`, not `PERMISSION_DENIED`: the
  caller has not been denied by policy; Chat cannot safely decide. This follows
  the repository error convention and explicit T-056-P2 direction.
- A missing checker is treated as unavailable in the eventual production path,
  not as “no peer deleted”. This intentionally changes the current optional
  collaborator semantics for this protected A1 path.
- Tests use existing gRPC methods `ListChats`, `CreateDM`, and `GetDM`; no
  proto/API extension is necessary for this server-side gate.

## Risks And Follow-Ups

- Filtering after a database cursor can produce sparse/empty pages. Tests must
  define the safe page-continuation behavior before implementation.
- The User profile lookup maps profile to Auth account. Its not-found/error
  contract needs a privacy-safe unavailable outcome for this feature.
- The A1 terminal localized marker remains a Flutter/API-signalling decision
  and is deliberately not solved by this Chat-only change.
