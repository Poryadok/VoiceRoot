# ExecPlan: ListThreads versioned read-model successor

## Purpose

Replace the limit-only `ListThreads` implementation with a bounded, per-viewer Messaging projection that serves a correct thread index for DM, group, and channel chats. A caller gets an opaque page chain with exact visible reply count, latest reply and preview; revocations made between pages cannot disclose private data or duplicate a thread parent.

## Context

- Docs: `docs/microservices/messaging-service.md` defines Messaging ownership, thread RPCs and `ghost_only`; `docs/DATA_STORES.md` requires database-per-service; `docs/ARCHITECTURE_REQUIREMENTS.md` separates Messaging history from Realtime; `docs/TESTING.md` defines TDD.
- Decision: `tmp/slave-driver/listthreads-successor-sol-contract.md` is the 2026-09-14 Sol architecture contract. It supersedes PR #364's SQL-candidate approach.
- Current code: `src/backend/messaging/internal/grpcsvc/messaging_grpc.go` calls the unbounded `MessagesStore.ListThreads`; `messages_store.go` aggregates `messages` on every request. The existing proto already supplies `CursorPageRequest` and `ThreadList.next_cursor`.
- Constraints: Messaging owns only `messaging_db`; Chat remains membership authority. No production fallback may use the legacy aggregate. Do not run Realtime tests, race tests, Compose, localhost harnesses, or Testcontainers locally.

## Scope

- In: durable membership inbox and per-viewer build lifecycle; journalled mutation projection; immutable dual AVL indexes and cursor states; `ListThreads` activation; schema, workers, observability, migration/backfill, docs and contract tests.
- Out: reuse, cherry-pick, or extension of PR #364; a candidate-limited SQL aggregate; global WebSocket history catch-up; cross-database foreign keys; changing Chat's membership authority.
- Documentation gap: Chat must expose the durable membership-outbox payload and delivery/retry guarantees. The Sol contract fixes required fields and semantics; the implementation must document its concrete Chat event transport before activation.

## Milestones

- [x] Record the successor contract and RED-only groundwork without production changes.
- [ ] Add `messaging_db` migrations and stores for viewer records, changes, immutable nodes, cursor states and membership inbox.
- [ ] Deliver durable Chat membership events and the Messaging consumer/backfill worker.
- [ ] Implement serialized, atomic projectors and immutable AVL traversal.
- [ ] Activate `ListThreads` after readiness, zero lag, invariant checks and sampled dual reads.
- [ ] Complete unit, integration, migration and hosted CI evidence; retire the legacy list path only after activation evidence.

## Detailed Steps

1. Add Messaging-owned schema, no cross-DB FK: `thread_list_viewers`, ordered `thread_list_changes`, immutable activity/id AVL nodes, `thread_list_cursor_states`, and deduplicating membership inbox. Add migrations and rollback/retention notes.
2. Define Chat's durable outbox event with `event_id`, `chat_id`, `profile_id`, monotonic `membership_revision`, and `joined|removed|left`; consume idempotently before building or revoking viewer state.
3. Build a viewer using a repeatable-read baseline, bulk-build both trees, replay changes after its baseline sequence, then CAS `BUILDING` to `READY`. New joins use the same path; removal is denied by Chat first and cleanup may lag.
4. Route every local root/reply/edit/delete/hide/ghost mutation through one transactional projector. Append its journal change with the message mutation, update current heads under advisory lock and revision CAS, and roll back the message when projection fails.
5. Make page one authorize with `EnsureMember` before any cursor work, capture READY roots and high-water state at the mutation serialization point, and traverse at most `O(log T + page_size + 1)` logical nodes. Use only the state ID plus HMAC-bound chat/profile/page size/snapshot/expiry in cursor v2.
6. Preserve the first cursor expiry for all pages. Make missing pre-expiry state and `BUILDING`/`UPDATING` projection availability fail closed with the opaque unavailable message; treat expired/tampered/mismatched cursors as invalid only after membership authorization.
7. Apply `FOR_ME`, `FOR_EVERYONE`, ghost and hide revocations to all unexpired relevant cursor states. A pending fan-out marks the state `UPDATING`; never return stale private data. New grants affect only new snapshots.
8. Add cursor-state and immutable-node GC after 15 minutes, retaining journal records through the smallest BUILDING baseline watermark. Mark/sweep only nodes unreachable from READY heads and unexpired cursor states.
9. Before activation prove zero projection lag, root invariants and sampled legacy dual reads. Keep observability for readiness, lag, projector error and state-GC health.

## Validation

- [x] `go test -tags listthreads_successor_red ./internal/threadlist` in `src/backend/messaging` demonstrates the intended RED state without starting a database/container: missing implementation symbols make the contract test fail to compile.
- [ ] Unit/property tests cover path-copy immutability, deterministic retry, AVL height/traversal bound and no `messages` read for a 100k-reply hot thread.
- [ ] Integration/migration tests cover DM/group/channel, ties, page-size binding, membership event replay/dedup/out-of-order/crash, BUILDING/READY/REVOKED, snapshot chains and every root/reply visibility revocation.
- [ ] Cursor tests cover HMAC binding, original TTL, missing state before expiry, expiry and opaque errors.
- [ ] Fault-injection tests prove mutation/projector atomicity and GC shared-node preservation.
- [ ] `make buf-ci`, `make buf-breaking`, `make buf-go-pb-check`, focused Messaging tests, migration checks and hosted CI pass on the final implementation PR.

## Progress

- [x] Read project, Messaging, data-store, test and Sol-contract sources.
- [x] Created clean branch from `72589878` without copying PR #364.
- [x] Added the versioned-read-model documentation, ExecPlan and tagged RED contract scaffold.
- [x] Fresh RED test review identified a required matrix beyond the first scaffold. The revised scaffold now encodes schema ownership, membership deduplication, path-copy/traversal, cursor binding/TTL, atomic rollback, fail-closed reader states, authorization order, live root/reply revocation and shared-node GC. Migration SQL, all-chat integration and fault-injection matrices remain explicit future RED cycles before GREEN.
- [x] Fresh re-review accepted the revised RED scaffold.
- [ ] Production implementation and GREEN cycle.

## Decisions

- Use the Sol persistent per-viewer versioned read model because exact per-viewer visibility, live revocation and 15-minute snapshot pagination cannot be bounded by the current `messages` aggregate.
- Keep `ListThreads` RPC wire fields additive for this slice; cursor v2 changes cursor content, not the protobuf shape.
- RED tests are build-tagged to keep the documentation/test-groundwork PR reviewable while deliberately exposing missing implementation with an explicit command. They run no containers or local services.
- The RED scaffold defines implementation seams only where required to make a test executable; it does not choose a Chat event subject, node serialization, proto addition, or cross-service adapter. Those details remain implementation design work under the fixed event payload/semantics.

## Risks And Follow-Ups

- The current Chat membership path lacks the required durable outbox contract; activation is blocked until it exists and Messaging consumes it idempotently.
- A failed or lagging projector must block the affected page rather than return stale data or fall back to SQL.
- The final migration and backfill plan needs production rollout evidence and a retention calculation for the oldest BUILDING baseline.
