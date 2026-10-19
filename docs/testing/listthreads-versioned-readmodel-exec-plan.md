# ExecPlan: ListThreads versioned read-model successor

## Purpose

Replace the limit-only `ListThreads` implementation with a bounded, per-viewer Messaging projection that serves a correct thread index for DM, group, and channel chats. A caller gets an opaque page chain with exact visible reply count, latest reply and preview; revocations made between pages cannot disclose private data or duplicate a thread parent.

## Context

- Docs: `docs/microservices/messaging-service.md` defines Messaging ownership, thread RPCs and `ghost_only`; `docs/DATA_STORES.md` requires database-per-service; `docs/ARCHITECTURE_REQUIREMENTS.md` separates Messaging history from Realtime; `docs/TESTING.md` defines TDD.
- Decision: `tmp/slave-driver/listthreads-successor-sol-contract.md` is the 2026-09-14 Sol architecture contract. It supersedes PR #364's SQL-candidate approach.
- Current code: `src/backend/messaging/internal/grpcsvc/messaging_grpc.go` authorizes then returns the unavailable readiness guard. The legacy `MessagesStore.ListThreads` aggregate remains only as predecessor code, not an RPC fallback. The existing proto already supplies `CursorPageRequest` and `ThreadList.next_cursor`.
- Constraints: Messaging owns only `messaging_db`; Chat remains membership authority. No production fallback may use the legacy aggregate. Do not run Realtime tests, race tests, Compose, localhost harnesses, or Testcontainers locally.

## Scope

- In: durable membership inbox and per-viewer build lifecycle; journalled mutation projection; immutable dual AVL indexes and cursor states; `ListThreads` activation; schema, workers, observability, migration/backfill, docs and contract tests.
- Out: reuse, cherry-pick, or extension of PR #364; a candidate-limited SQL aggregate; global WebSocket history catch-up; cross-database foreign keys; changing Chat's membership authority.
- The effective-membership source, admission and backfill contract is defined below. Its owner APIs, durable bridge and concrete event transport must be implemented and verified before activation.

## Milestones

- [x] Record the successor contract and RED-only groundwork without production changes.
- [x] Independently review the BE-183 source-consistency protocol and record the effective-membership contract v1 and owner obligations below.
- [ ] Add `messaging_db` migrations and stores for viewer records, changes, immutable nodes, cursor states and membership inbox.
- [ ] Deliver durable Chat membership events and the Messaging consumer/backfill worker.
- [ ] Implement serialized, atomic projectors and immutable AVL traversal.
- [ ] Activate `ListThreads` after readiness, zero lag, invariant checks and sampled dual reads.
- [ ] Complete unit, integration, migration and hosted CI evidence; retire the legacy list path only after activation evidence.

### BE-184 persistent-index implementation checkpoint

The first implementation slice is the dual immutable AVL primitive consumed by
the BE-184 transactional viewer projector, and then BE-185's page reader. It
stores exact per-viewer summaries in content-addressed Messaging-owned nodes;
activity order is `(last_reply_at DESC, parent_id DESC)` and a second parent-ID
tree supports logarithmic replacement/removal. Updates path-copy both roots.
The SQL adapter accepts the same pgx transaction as future journal/root writes.
Migration `000031_thread_list_nodes` rejects in-place SQL updates; deleting nodes
is reserved for future reachability GC. No runtime RPC or authorization path is
enabled by this checkpoint.

Acceptance cases: old roots remain unchanged after upsert/delete; identical retry
keeps the roots; randomized edits match a sorted oracle; adversarial sorted
insertion remains balanced; each page reads at most height + page size nodes;
100k replies remain one summary; missing/corrupt nodes fail closed; real SQL
rejects updates and rolls back all inserted nodes with the owning transaction.
Use owned Linux Go 1.26 + PostgreSQL 16 without exposed host ports. Ordinary TDD;
migration/privacy/concurrency risk requires independent review. Source-revision
APIs/outboxes, Q/J journals, durable roots/build/GC, all visibility projectors and
complete admission remain subsequent BE-184/185 work before activation. Do not
mark the full successor or task complete from this primitive's checks.

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

- The current Chat membership path lacks the required durable source/export implementation; activation is blocked until the contract below is implemented and Messaging consumes it idempotently.
- A failed or lagging projector must block the affected page rather than return stale data or fall back to SQL.
- The final migration and backfill plan needs production rollout evidence and a retention calculation for the oldest BUILDING baseline.

## Effective-membership contract v1

This is the reviewed BE-183 technical contract for BE-184/185, not a claim that
the bridge or read model is implemented. Chat owns effective membership;
Messaging owns its projection. Keep `ListThreads` unavailable until the complete
protocol and all existing message-visibility gates are verified. No direct reads
of another service's database, fixed first-N admission or SQL fallback qualify.

### Source revisions and complete snapshots

Every source coordinate is `(owner, scope, incarnation, revision)`. Revisions
never decrease or repeat within an incarnation; loss of journal continuity or
identity reset creates a new incarnation. Each owner commits state, its serialized
head increment, exact replay record and outbox entry in one local transaction.
Use a locked head or another proved committed prefix, never `MAX(sequence)`,
allocation order, wall-clock time or broker delivery sequence. Rollback cannot
leave a committed head without its event. Every mutation, batch, cleanup and
lifecycle writer affecting the scope participates.

| Owner / scope | Inputs covered by the revision |
| --- | --- |
| Chat / chat | Local membership, DM `deleted_for_self`/actual reopen, Space binding, resource lifecycle/fences and authority-changing managed membership |
| Space / Space | Complete membership, join/leave/kick/ban and lifecycle/allowed restoration |
| Role / Space | Every `TEXT_CHAT_VIEW` input: role assignment, permission bits, text overrides, Owner/Admin effects, ownership prepare/finalize/abort and lifecycle fences |
| User / profile | Eligibility and all deleted/disabled/frozen causes, including durable negative/tombstone lookup |

Each scope also persists monotonic `revocation_revision`. Every potentially
restrictive change increments it; uncertainty is restrictive. Grant-only changes
still increment the ordinary revision. Broad Role edits may conservatively fence
the entire Space; narrower invalidation requires complete affected-viewer proof.
This counter preserves remove/rejoin and deny/allow evidence even when snapshots
coalesce intermediate states; it does not become a permission authority.

Replay binds event ID, source coordinate and payload hash. Identical duplicates
are idempotent; conflicting bytes for the same identity/revision quarantine the
scope. A missing or unapplied tail is not caught up. A sealed complete snapshot
can repair a gap only while preserving the revocation counter. Unknown revocation
history requires a new incarnation and invalidates previous cursor lineages.

Owners expose immutable paged snapshots with sealed heads, or atomic state/head
reads followed by authoritative head validation. Every page binds scope,
incarnation, snapshot ID and position; explicit terminal completion proves full
enumeration. Empty pages and transport success alone do not. A registered
consumer/build pin retains snapshots and replay history until released; expiry
forces a fresh baseline rather than best-effort continuation.

Chat captures its local definition and complete local candidates or complete
Space member set, thereby discovering every dependency scope. Role evaluates
`TEXT_CHAT_VIEW` for that exact set at a pinned revision; User supplies eligibility
for every candidate at pinned per-profile revisions. Role remains the permission
calculator. Results include all actual ownership/lifecycle dependencies.

After **all** state reads/pages finish, Chat authoritatively rereads **every**
dependency head. Accept only identical incarnations/revisions and complete
coverage, otherwise retry within a bounded budget then return unavailable.
State/head reads serialize with owner writes; stale allow caches or lagging
replicas cannot validate. An unchanged interval for each dependency overlaps
between the last state read and first validation, proving a common historical
cut. A changed Chat/Space head invalidates the candidate/dependency set too.
Point admission needs only the exact viewer's dependencies. This cut is not an
ongoing permission grant: later pages still require fresh admission.

### Protected internal API obligations

These are required additive API capabilities, not existing callable RPCs. The
implementation freezes their protobuf full method names, request/response fields
and caller allow-lists before enabling them. All use the signed-principal
[Phase-0 contract](../ARCHITECTURE_REQUIREMENTS.md):
exact issuer/subject, audience, full RPC, request ID and deterministic protobuf
request hash; bounded credential lifetime, TLS/JWKS and replay checks. Raw caller
metadata and public viewer-filtered profile APIs grant no authority.

| Caller → owner | Permitted capability and binding |
| --- | --- |
| `chat` → `space` | Complete member snapshot, replay/head validation and retention pin for the exact Space |
| `chat` → `role` | Pinned effective `TEXT_CHAT_VIEW` decisions and head validation for exact Space/chat/candidate IDs |
| `chat` → `user` | Exact profile eligibility/tombstone revision and head validation; no public profile disclosure |
| `messaging` → `chat` | Complete effective-member generation/export, replay/head/pin and request-bound exact `(chat_id, profile_id)` admission |

Snapshot/pin/continuation requests bind their exact scope, handle, position and
expected source coordinates; no caller may cross scopes through a cursor or
retention handle. Admission returns decision, source vector/revocation fences,
lineage and required committed Chat journal offset. A missing/unreconciled source
or lifecycle freeze is unavailable; a known non-member/ineligible viewer is
denied. No Gateway/public route is added. These callers do not inherit the
Auth-only `GetSdkProfileEligibility` grant. Auth still validates account/session
state; User's delayed account overlay cannot replace it.

### Chat generation and durable journal Q

Chat persists complete effective-member generation `G`, its validated vector,
candidate coverage, decisions and revocation fences. A single `chat_db`
transaction checks the local Chat revision, activates `G`, updates pair states
and appends committed per-chat prefix `Q`. Each pair has monotonic
`membership_revision` and admission lineage. Effective false→true emits `joined`;
true→false emits `removed|left` with the actual reason. Preserve the accepted
`event_id`, `chat_id`, `profile_id`, `membership_revision`, `kind` payload and add
lineage, source fences and journal offset. No-op refresh can advance a barrier
without fabricating a join. One DM side's denial does not remove the peer.

Potential-revocation barriers are durable even if recomputation ends allowed.
They block older lineages until reconciled. Confirmed revoke/regrant creates a
new lineage; conservatively retire the old lineage whenever continuously allowed
intermediate states cannot be proved. An old cursor never silently restarts or
revives; before expiry it retains the opaque unavailable result.

Chat exports immutable `G` plus `Q`, complete pages and replay strictly after
`Q`, with retention registered before export. Missing required history expires
the export. Later external changes still require fresh point admission and
reconciliation. Messaging commits inbox deduplication, events/barriers and viewer
state together before ACK. Duplicate/reordered events cannot resurrect revoked
lineages; a high pair revision is not proof of a complete chat prefix. Track `Q`
or an explicitly sealed complete per-viewer range separately.

### Messaging baseline J, READY and page admission

Chat membership prefix `Q` and Messaging message/visibility journal `J` are
different clocks; never compare or collapse them. Persist
`BUILDING(chat, viewer, lineage, G, Q)`, pin retention, take a repeatable-read
Messaging baseline with rows and committed `J` in the same snapshot, bulk-build
both immutable trees and replay local changes after `J` plus membership/barriers
after `Q`.

Finalization obtains fresh authoritative exact-pair Chat admission naming current
vector, lineage and required `Q`. Within Messaging's local serialization
transaction, require fully consumed `Q`, an allowed matching lineage, no fan-out
or replay gap, and `J` equal to the current committed local head. CAS the build
generation/revision to READY with roots and both watermarks. A late worker cannot
overwrite a newer revoke/lineage. READY proves local readiness, never cached
authorization or a globally atomic permission decision.

Every page first obtains exact-pair Chat admission **before cursor parsing**,
including malformed cursors. Only READY roots/cursor states matching lineage and
consumed required `Q` may be read. Stage the bounded tree result, then obtain a
second fresh Chat proof before cursor progress commits or bytes return. Changed
relevant vector/fence or denial discards the result. A local version/CAS check
also proves no Messaging visibility mutation or `UPDATING` barrier committed
between capture and final proof. Bounded retries end in the fixed unavailable
response, without partial metadata.

This orders an overlapping revoke at the final completed permission decision,
as in the existing Role authorization contract. A revoke committed before the
call or between pages is detected even with outbox delivery stopped. A
revoke/regrant cannot restore an old lineage. Already authorized overlapping
requests may finish; the protocol makes no zero-network-bytes-after-revoke
guarantee. Original 15-minute cursor expiry, immutable traversal bounds and
opaque unavailable/invalid error distinctions remain unchanged.

### DM, profile and message visibility boundaries

- DM `deleted_for_self=true` retires only that viewer's lineage atomically with
  Chat's state/outbox. An actual existing reopen creates a fresh lineage and
  BUILDING state; no new automatic reopen trigger or history truncation is added.
- Chat archive is navigation state and preserves ordinary history access.
  Deleted/archived profiles are ineligible as viewers; disabled/frozen causes
  must all be cleared by their owner before eligibility returns. Profile switch
  and presence do not revoke another profile. Archived domain data is retained.
- Deleting a peer/author does not remove the surviving viewer's known DM history.
  Auth account/session revocation remains separate; Chat's fresh-inbox peer
  filter does not become a history policy.
- Membership does not replace per-message directional Social blocks, hides,
  ghost visibility, managed message-time entitlement intervals or deletion
  fences. Their projection and cursor invalidation remain activation gates.

### Consumer implementation and adversarial proof

BE-184 implements owner revisions/outboxes/snapshot APIs, complete Chat
reconciliation/export and Messaging inbox/build/AVL/journal/GC. BE-185 replaces
first-100 admission and wires page proofs/cursor bindings/activation. Contract
publication may unblock only the explicitly pinned implementation gate; it does
not satisfy deployment, acceptance or full-done prerequisites.

Required GREEN evidence supplements the original validation matrix:

1. Transaction rollback/crash; conflicting duplicate hashes; reordered/missing
   tails with a silent broker; retention expiry, reset and incarnation mismatch.
2. More than 100 members and multi-page snapshots; candidate insertion during
   enumeration; concurrent Role/User changes; reject a fractured cut where Space
   allows before removal but Role allows only after removal.
3. Revoke during BUILDING, between proof and READY CAS, between pages and during
   staged reads; delayed/coalesced revoke/regrant; stale worker completion;
   previous cursor lineage never revives.
4. One-side DM delete/reopen, archive versus profile delete, independent freeze
   causes, and peer deletion preserving surviving known history.
5. Missing Role, unavailable/stale authority, wrong signed caller/request/scope
   and malformed cursor: authorization first, opaque failure and no legacy read.
6. Real persistence tests for atomic message/visibility journals and barriers,
   zero-lag READY, immutable AVL bounds, original expiry and shared-node GC.

## Readiness containment implementation plan

The full successor remains open. The currently callable aggregate ignores viewer
hides and ghost visibility and cannot satisfy the accepted fail-closed contract.
Until a verified projection exists, the RPC must authorize the caller and return
`UNAVAILABLE "thread list unavailable"` without querying that aggregate. This
temporarily makes the thread index unavailable; sending replies and loading a
known thread through `GetThreadMessages` are outside this change.

Sources inspected: the service contract above; `docs/todo/backend.md`;
`docs/features/text-chat.md`; `docs/DATA_MODEL.md`; `docs/DATA_STORES.md`;
`docs/TESTING.md`; Chat's `store/space_members.go` and
`grpcsvc/space_membership.go`; Messaging's `ListThreads` handler and store.

1. Delegate handler RED tests for authentication, chat validation, mandatory
   membership authorization, denial before malformed cursor handling, opaque
   dependency failures, context cancellation/deadline and unavailable results for
   DM/group/channel with either empty or nonempty cursors. A zero-value store
   requires no database; code review must separately prove no legacy invocation.
2. Review assertions against the accepted missing-projection contract. Implement
   the bounded readiness guard, preserving specific authorization denial and
   cancellation codes; log concrete dependency diagnostics only server-side.
3. Run focused tests, `go test -short ./...`, `go vet ./...` and pinned
   `golangci-lint run ./...` in Messaging. Use hosted integration CI for affected
   components; no local service/container harness is needed for this guard.
4. Obtain independent security review, update evidence, push and merge only with
   successful CI on the current head. Keep the successor TODO and all production
   projection milestones unchecked.

The effective-membership contract above resolves source revisions, complete
enumeration/watermarks, DM delete/reopen invalidation and profile eligibility.
The current bridge still lacks that implementation; a trigger on `chat_members`
alone cannot durably enumerate Space/Role-derived viewers. The Messaging Chat
adapter still reads only the first 100 members. Replace it with exact-pair
authoritative admission, implement source recovery and verify every activation
barrier before enabling the full successor. Transport subject/table/backoff is
routine implementation work under this contract. Keep archived domain data intact.

Before the full successor GREEN cycle, replace the tagged RED scaffold's
disconnected fixture seams with actual persistence tests: cursor registration
and target binding, paused versus completed visibility fan-out, SQL immutable
node enforcement, adversarial AVL traversal, persisted transaction rollback and
real shared-node GC reachability. No production globals or constant metadata
helpers may be introduced merely to satisfy that scaffold.

Containment evidence:

- [x] Focused handler RED demonstrated legacy `Internal` errors instead of
  unavailable/permission/cancellation/configuration outcomes; the same tests
  pass after the guard (`go test -short ./internal/grpcsvc -run ListThreads -count=1`).
- [x] Independent test review accepted the matrix and corrected the no-store-call
  evidence claim. Test-author delegation was unavailable due to the agent limit;
  the lifecycle owner authored the tests before implementation.
- [x] Independent security review confirmed no aggregate invocation, mandatory
  membership check, opaque dependency errors and preserved cancellation codes.
- [x] Messaging `go test -short ./...` and `go vet ./...` pass.
- [ ] Hosted CI on the final head and merge commit.
