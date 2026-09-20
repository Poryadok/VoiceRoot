# ExecPlan: A1 soft-delete Quick Access recovery

## Purpose

Close the A1 snapshot leak where a DM whose peer account was soft-deleted can
still appear in the caller's fresh Quick Access response. A successful
`ListQuickAccess` response must omit it, just as `ListChats` already omits it
from main, requests, archive, and folders.

## Context

- Docs: `docs/PLAN.md` (A1), `docs/ARCHITECTURE_REQUIREMENTS.md` (REST
  snapshots and selected-chat history), `docs/features/text-chat.md`,
  `docs/microservices/auth-service.md`, `docs/microservices/chat-service.md`,
  `docs/microservices/messaging-service.md`, `docs/microservices/realtime-service.md`,
  `docs/microservices/social-service.md`, `docs/microservices/user-service.md`,
  `docs/DATA_MODEL.md`, `docs/DATA_STORES.md`, `docs/TESTING.md`, and
  `docs/CONTRIBUTING.md`.
- Explicit A1 contract supplied for this recovery: Auth deletion and session
  epoch revocation fail closed; neither direction may create a DM or send a
  message after deletion; fresh user-visible snapshots omit a deleted-peer DM
  from main, requests, archive, folders, and Quick Access. A known selected DM
  retains history and exposes deleted-peer state. Client terminal markers are
  in-memory only; Realtime accelerates REST recovery. Restore/purge/tombstones
  and attachment GC are A4/out of scope.
- Code: `src/backend/chat/internal/grpcsvc/list_chats_deleted.go` supplies the
  existing fail-closed lifecycle filter. `quick_access.go` currently loads and
  hydrates every stored shortcut without applying that filter.

## Scope

- In: make Chat `ListQuickAccess` use the same Auth/User lifecycle lookup and
  fail-closed deleted-peer filtering as `ListChats`; prove deleted peers are
  omitted and an unavailable lifecycle gate does not leak a shortcut.
- In: document this narrowly scoped closure and its remaining A1 dependencies.
- Out: Auth deletion endpoint/session work, `CreateDM`, Messaging send/history,
  Realtime, Flutter terminal markers, restore/purge/tombstones, attachment GC,
  and any live Compose/race/localhost test.

## Milestones

- [x] Recover in an exclusive clean Treehouse lease at base `7326bd05`; do not
  access slot 2.
- [x] Locate the bypass: `ChatGRPC.ListQuickAccess` does not call
  `filterDeletedPeerDMRows`.
- [x] RED: added a focused Chat gRPC integration test for an active plus
  deleted-peer Quick Access DM. The no-Docker unit contract failed because the
  Quick Access lifecycle-filter method did not exist.
- [x] GREEN: applied the existing lifecycle filter before Quick Access response
  hydration and made lifecycle lookup failure return `Unavailable`.
- [x] Refactor/verify: formatted the Go files; focused unit and Chat short
  suite are green. The database integration proof requires hosted CI.
- [ ] Commit, push, PR, hosted CI/`ci-gate`, merge commit, refresh and verify
  `master`, then return the lease.

## Detailed Steps

1. Seed pre-delete DMs directly in the Chat integration fixture, add both to
   Quick Access, then start Chat with the deleted-account checker. Assert only
   the active peer is visible. Add a separate unavailable-checker assertion.
2. Run that test before production edits and record the expected RED leak.
3. Extend the Quick Access store boundary only as needed to retrieve DM peers
   for the shortcut rows. Reuse `filterDeletedPeerDMRows`; do not introduce a
   client-side heuristic or a fail-open fallback.
4. Filter rows before `FindChatByID` hydration. On a missing peer mapping,
   User lookup, or Auth deletion lookup failure, return `Unavailable` and no
   response body.
5. Run the focused test again, then `go test -short ./...` in Chat if local
   platform dependencies permit. Do not run Windows-local Compose, Realtime,
   race, or localhost runtime tests.
6. Update this plan's Progress/Decisions with exact command evidence, inspect
   the diff, and complete the normal PR lifecycle.

## Validation

- [x] RED: `go test ./internal/grpcsvc -run
  TestQuickAccess_DeletedPeerFilterUnit -count=1` failed with the expected
  missing `filterQuickAccessDeletedPeerDMs` method before production code.
- [x] GREEN: the same command passed after implementation; additionally,
  `go test ./internal/grpcsvc -run
  "Test(QuickAccess_DeletedPeerFilterUnit|ListQuickAccess_DeletedPeerFilterUnit)" -count=1`
  passed and proves the RPC filters before serializing its response.
- [x] `go test -short ./...` from `src/backend/chat` passed.
- [ ] `go test ./internal/grpcsvc -run TestQuickAccess_HidesDeletedPeerDMAndFailsClosed -count=1`
  is blocked locally because Testcontainers panics on Windows rootless Docker;
  hosted CI remains required.
- [ ] Hosted PR CI reports success for the exact head, including `ci-gate`.

## Progress

- [x] Read the governing A1 and service/testing/git documentation and the
  recovery state checkpoint.
- [x] Acquired lease `32a78901b86e1dc8d786cb98e00d57f3` in slot 1 and created
  `feature/a1-soft-delete-quick-access` from `7326bd05`.
- [x] Tests first: added an externally observable PostgreSQL integration test
  and an executable no-Docker unit boundary test.
- [x] Completed RED→GREEN: missing Quick Access lifecycle filter → fail-closed
  filter that omits deleted-peer DMs while retaining active entries.
- [x] Local validation: focused unit test and `go test -short ./...` passed.

## Decisions

- Reuse `filterDeletedPeerDMRows`: it already embodies the required internal
  lifecycle-owner lookup and Auth fail-closed policy used by every `ListChats`
  snapshot scope.
- Keep stored Quick Access rows intact: the A1 contract requires their absence
  from a fresh user-visible snapshot, not a destructive deletion of user
  navigation state.
- Classify every stored shortcut before disclosure and reject an unavailable
  target/peer lookup: a stale row must not become a fail-open identifier leak.

## Risks And Follow-Ups

- This atomic PR closes only the Quick Access snapshot bypass. The other A1
  contract paths are owned by their corresponding Auth, Chat/Messaging,
  Realtime, and Flutter verticals.
- Chat integration tests use PostgreSQL/Testcontainers. If Docker is not
  available locally, hosted CI is the required environment for that evidence.
