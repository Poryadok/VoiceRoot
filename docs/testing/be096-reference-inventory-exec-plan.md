# BE-096 Story reference inventory and protective File import

## Purpose

Preserve historical and pending media pins while the Story/File reference ledger
is reconciled. The concrete consumer is Story expiry/deletion and recovery
(BE-096/BE-260). This first slice must not authorize physical deletion or claim
global reference activation.

## Context

The technical design is D116's accepted File reference migration v1, BE-096 c17,
attachment `aa1c6025f63cd567bae8d6157ddb324b`, SHA256
`d6af34a6c109f0f95ef82a9878ef04db7c545050ffe9c8b270a86c6d33b31701`.
It implements the exact-reference/global-zero rule in DATA_MODEL.md and the
all-owner backfill/catch-up/fence/reader requirements in file-service.md. It does
not replace those requirements with a Story-only seal.

Baseline `54927c5344f94a6be02bb6aba1f48852944588e1` combines PR982
`8c1f6298bce71820dd04db0967264b43bd9a53cb` and PR1010
`f3d6ef4b6da649837cfe08476b4639e1d4f0b870`, retaining both signed reference calls
and the mTLS client certificate. Author Dimas-43 uses a fresh managed lease and
`feature/be096-story-inventory-import`; the reviewed branches remain immutable.

## Scope

In: dedicated typed inventory wire; File-only authenticated Story export;
transactional owner counter and journal; immutable materialized snapshot pages;
File direct pull, immutable import receipts, counted conflicts and private blob
holds; an incomplete-snapshot destruction barrier at actual cleanup/GC paths.
Global provider remains nil. Historical import does not call ClaimStoryMedia,
create a readable reference, revive a terminal row, or grant a URL.

Out: other-owner implementations, catalog/cover policy invention, global ACTIVE
provider, external non-rollback restore controller, production writer drain,
reader cutover, DEV/STG, rollout or acceptance. Other required coverage components
remain MISSING. Independent highlight-cover ownership is counted unresolved.

## Milestones

- [x] Accepted design, custody and coherent combined source established.
- [ ] Typed deterministic inventory format and strict validation tests.
- [ ] Story transactional gate/counter, retained+pending+unresolved materialization,
  immutable pages and complete absolute-state journal revisions.
- [ ] File-only protected exporter and real request-bound pull client.
- [ ] File barrier, immutable import/cursors/conflicts and protective holds;
  actual GC claim/pre-DELETE and alternate destructive paths respect them.
- [ ] Real PostgreSQL concurrency, replay, migration rollback and protected
  transport checks, followed by independent frozen-diff review.

## Detailed Steps

1. Define protocol1 identity with installation, restore cohort, migration, File
   incarnation, source service/component, owner incarnation, writer version and
   snapshot UUID. Pages are contiguous, fixed at at most1000 entries, ordered by
   raw UUID tuple bytes. Typed page commitments and manifest use P3 deterministic
   protobuf/domain hashes. Unknown nested fields, invalid UUIDs, enum values,
   duplicate reasons/operations, reordered tuples or changed replay bytes deny.
2. Lock Story's singleton gate before ownership rows in every relevant mutation.
   Advance its transactional counter with complete indexed journal revisions in
   that transaction; rollback consumes no committed revision. Materialize under
   the same gate, then serve immutable stored pages rather than moving tables.
   Union extant logically retained Story media, creation intents and release
   obligations; include every reason and operation. Count historical text/LFP
   anomalies, tuple conflicts and unresolved independent covers.
3. Add service:File-only export with exact Story audience/RPC/body/replay binding
   and mTLS. No forwarded profile headers or Gateway routes. A writer seal cannot
   assert complete replica/direct-writer coverage that has not been proved.
4. Enter File's durable migration barrier before fetching an owner snapshot.
   Fetch remotely outside File transactions. Under cohort/operation/ordered
   File/blob locks, atomically persist immutable page receipt, hold/issue rows and
   cursor. Terminal/missing/conflicting/retired rows stay unresolved; never infer
   publication admission from a historical row. Pending/negative journal states
   cannot drop the final protection while migration is incomplete.
5. Guard actual GC claim AND second pre-DELETE transaction, plus uploader,
   expiry, catalog and derivative cleanup/alias retirement, against the same
   cohort and holds. Preserve durable ambiguous-delete intents and immutable key
   generations. Reconcile File/blob lock ordering before importer races are run.
6. DOWN locks relevant tables before checks and rejects retained snapshots,
   journals, holds, conflicts or cohort fences. Completed-record30-day cleanup
   must not erase incomplete/unacknowledged work or current proof.

## Validation

Use TDD for behavior. Required discriminators: historical highlighted S0 without
ledger + new S1 shared blob; delayed snapshot while actual GC runs; absent other
component cannot activate; owner rollback/commit ordering; immutable page retry
after live mutation; missing/overlap/conflicting page and partial revision denial;
pending remote acquire/lost owner commit; missing/terminal File and cover anomaly
remain counted; importer grants no URL; holds versus irreversible GC claim;
cohort invalidation versus claim/pre-delete; private signed/mTLS caller failures;
guarded DOWN under concurrent writers. Positive global deletion is deliberately
unproved, not simulated with a fixture production authority.

Before each bounded Linux Go+PostgreSQL run inspect actual Docker CPU/RAM and
container occupancy. Use owned containers/caches only; never clean unknown jobs.
Run affected short/integration/race/lint/vet and Buf/generation checks. Preserve
terminal handles and portable TT evidence. Graphify AST refresh is an independently
owned closeout operation, not a runtime activation proof.

## Progress and Decisions

Contract and first source boundary accepted by D116; no new product retention or
role authority is inferred. The ctx-only release provider stays nil; eventual
activation requires a caller-transaction cohort lock and all-owner/read/fence
proof. Snapshot import is protective, not current-publication ClaimStoryMedia.

## Risks and Follow-Ups

External restore fencing and unsettled object-store requests remain real
production dependencies. Catalog/pack and independent cover tuple shapes, A7
retention-account ownership, remaining owner inventories, replica/direct-writer
exclusion and reader cutover remain explicit global blockers. No Story-only
snapshot or boolean closes them. Root routes the independent review after the
exact source/head and terminal validation packet is frozen.
