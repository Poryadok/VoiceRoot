# Realtime P3 durable admission execution plan

This plan implements TT BE-A2-REALTIME-P3-PARTICIPANT and accepted decision
BE-REALTIME-P3-DURABLE-ADMISSION-v1 (TT c3-c5). Sources: project AGENTS,
docs/PLAN.md (A2), realtime-service.md, space-service.md, DATA_MODEL.md,
DATA_STORES.md, ARCHITECTURE_REQUIREMENTS.md, CONTRACT_MATRIX.md, TESTING.md
and CONTRIBUTING.md. Current source base: d4abb829622e9bd91b26842b61522e6840840867.
Author: Рудеус-172. Independent concurrency review and delivery remain required.

## Safety contract

Realtime owns metadata-only PostgreSQL state. Every chat-scoped local enqueue
requires a committed admission tied to a never-reused process incarnation.
Admission remains PENDING until the last synchronous sink returns. SQL backend
loss, process timeout, cancellation and replacement do not settle it. FROZEN
closes new admissions, then persists a receipt only after all existing admissions
for the exact immutable Chat manifest have settled. PURGED is permanent.
Already queued frames are outside the boundary; no wire recall is promised.

Missing state is UNKNOWN. Only a protected Chat-native lifecycle resolver may
bootstrap it; delayed bootstrap cannot reopen a restrictive gate. All ordinary
DM/group and Space sinks use this route. Existing recipient authorization stays
in force. Lifecycle failure also denies sender delivery. Non-chat control events
retain existing semantics.

Import validates exact Chat pages/root, seals complete coverage and preserves
stable request/receipt bytes. Participant11 is additive; Space's existing BE005
consumer must wait for all11 FROZEN and PURGE_DECIDED barriers. LIVE restoration
requires complete prior freeze and an allowed newer generation. Dead-process
recovery without qualified exact termination proof stays unsupported/pending.
No timeout reaper or privileged caller boolean is introduced.

## Implementation sequence and proof

1. Risk-first real PostgreSQL regression: commit admission, pause sink, drop SQL
   backend, close gate, prove receipt pending; settle after sink, prove drainage.
   Check unrelated chat, restart incarnation, unknown state and monotonic hydrate.
2. Metadata schema/store and synchronous admission scopes. No payloads, recipient
   history or delivery queue. Lock order incarnation, operation, sorted chats,
   admissions; no RPC or sink IO in SQL transactions.
3. Add protected realtime wire and narrow Chat resolver, generated outputs and
   exact principal/listener negative tests. Use current mTLS/replay conventions.
4. Route real hub/profile/Redis/message/chat/ephemeral sinks through typed origin
   and admission; real composition tests prove no path bypasses lifecycle gate.
5. Existing Space importer/registry/transport/barriers accept participant11.
   Ten-good/missing11 cannot schedule, destructively purge or finish.
6. Canonical docs/store inventory/readiness and migration/rollback negatives.
   Run Linux package/race/PG tests, buf/codegen/drift and affected module checks.
   Bind JSON case counts and zero required skips to immutable source.
7. Independent security/concurrency review; deliver immutable source and portable
   proof to integration ant121 via accepted transfer, retain custody otherwise.

## Progress

- [x] Atomic TT claim; owned WT41 lease; current develop branch; source inventory.
- [x] Accepted Astra decision read; risk and consumer recorded in TT c7.
- [x] Initial missing-implementation compile RED; metadata implementation and
  real PG/race GREEN for five cases. The first PG-kill fixture killed an idle
  backend; it is superseded by MaxConns=1/acquire-after-Begin plus independent
  killer, which targets the backend that actually committed the admission.
- [ ] Protected wire, Chat bootstrap, all sinks, Space11 consumer, readiness/docs.
- [ ] Scoped verification, independent review, immutable delivery.

### Current implementation checkpoint (not ready for integration)

The real hub now uses committed admission for every chat-scoped sink, including
profile/Redis/ephemeral routes. Native Chat resolution cannot grant recipient
ACL. Sender fallback on failed Chat type lookup is removed. Exact manifests,
strong receipts and terminal denies are transactional; the Space importer,
registry, protected transport and terminal SQL predicate require participant11.
Space migration34 is additive to separately owned32/33; no membership triggers
or RoomClose hooks are changed. Schema is under migrations/realtime_db.

The actual hub+PG race run passed, including blocked recipient sinks, captured
late scope and stale LIVE hydration. The new 30-day compaction test is running;
full bytes are discarded while immutable hashes, denies and pending tickets
survive. Cooperative incarnation close exists; dead-process recovery remains
unsupported. Local Compose gains an independent PUBLIC-only HTTPS JWKS service
derived by the existing initializer from the same current/next signer keys,
avoiding application bootstrap cycles. Partial principal volumes fail startup.
Compose config parses; initializer image build passed. Bootstrap HTTPS runtime,
new protected listener composition, Chat native PG cases, Space34 SQL proof,
canonical docs, generation/drift checks and independent review remain open.

Owned resources: WT41 lease f8e38cf028e75e95fa9ac18395adee7d; Docker network
rudeus172-p3, PG rudeus172-p3-pg, build cache rudeus172-p3-build; no host ports or
shared stands. Preserve source/CI custody with actor172; ant121 publication only.

### Historical initial checkpoint (superseded by current implementation)

The metadata admission store/schema and synchronous scope executor exist; five
tests execute successfully with `-race -tags=realtimep3integration` in owned
Linux Go1.26/PostgreSQL16, zero skips. Exact test JSON is preserved with the task
evidence. The tagged PG suite intentionally requires its database instead of
silently skipping. Existing short Chat gisprincipal/grpcsvc/store checks pass,
which proves compilation/regression only, not new protected-listener runtime
coverage. `buf lint` passes. Native Windows tests were not executed.

Additive source includes participant11, separate `chat_lifecycle.proto`, native
Chat enrollment store/handler/listener/config and realtime lifecycle wire/stubs.
Native terminal `ChatLifecycleState` is distinct from the common fence-command
enum: common commands intentionally have no PURGED command. Chat's existing
schema is read under its existing mutation barrier; this adds no Chat migration.

**Do not integrate this partial package.** Next implementation is exact manifest
schema/import/seal/strong receipt finalization (same locked transaction as
drainage), protected Realtime listener + trusted Chat client/startup, every real
typed-origin sink and sender fail-closed handling, existing Space11 transport,
importer/registry/barriers and schema/config/readiness/canonical docs. Then add
new Chat protected composition/PG tests and complete the finite decision matrix,
required generation/drift checks and independent security/concurrency review.

Repeated mandatory CodeGraph follow-ups became unbounded and were cancelled as
owned read requests; no duplicate probe, index mutation or foreign process kill
was performed. Current exact-source reads controlled corrections. This is a
safe source/proof checkpoint, not completed task, public P3 capability, CI,
deployment or acceptance. Task/delivery custody stays with Рудеус-172 until an
explicit accepted real-ant transfer.

## Resource and activation boundaries

Use owned Linux Docker runners without host ports; no Windows test executable,
new host listener, firewall changes or shared container mutation. DEV/STAGING
claims, rollout and actual acceptance are separate; no such authority is claimed.
Public P3 readiness requires migrated/restorable realtime_db, trusted bootstrap
coverage, all actual/old writers drained or terminated, retained permanent denies,
11-participant release coverage and a qualified crash recovery path. A healthy
new binary or environment flag is not proof. Rollback cannot remove terminal
fences or introduce an old bypassing writer.
